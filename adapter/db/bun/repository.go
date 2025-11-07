package bunrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	pkg "github.com/smilemakc/mbkit"
	mberr "github.com/smilemakc/mbkit/adapter/db/errors"
	port "github.com/smilemakc/mbkit/application/service/port"
	"github.com/smilemakc/mbkit/domain/filters"
	"github.com/smilemakc/mbkit/internal/l"

	"github.com/uptrace/bun"
)

type IDLike = pkg.IDLike

type (
	GetterArgs                       = port.GetterArgs
	Saver[T any]                     = port.Saver[T]
	Getter[T any, ID pkg.IDLike]     = port.Getter[T, ID]
	Lister[T any]                    = port.Lister[T]
	Updater[T any, ID pkg.IDLike]    = port.Updater[T, ID]
	Deleter[ID pkg.IDLike]           = port.Deleter[ID]
	BulkDeleter[T any]               = port.BulkDeleter[T]
	Repository[T any, ID pkg.IDLike] = port.Repository[T, ID]
	Middleware[T any, ID pkg.IDLike] = port.Middleware[T, ID]
	Func[T any, ID pkg.IDLike]       = port.Func[T, ID]
)

// Deprecated use CustomFieldGetter instead.
type FieldGetter[T any] interface {
	GetByField(ctx context.Context, tx bun.IDB, field string, value any) (*T, error)
}

// Deprecated use CustomFieldGetter instead.
// Finder defines the ability to fetch an entity by an arbitrary field.
// Useful for cases when the primary key is not enough (e.g., lookup by email).
type Finder[T any] interface {
	// GetByField fetches a single entity where `field = value`.
	// Field must be explicitly allowed by implementation to prevent SQL injection.
	GetByField(ctx context.Context, tx bun.IDB, field string, value any) (*T, error)
}

type BaseGetter[T any, ID pkg.IDLike] struct {
	strict  bool   // if true, return the error (sql.ErrNoRows) if entity not found
	idField string // name of the ID field
}

func (b BaseGetter[T, ID]) baseQuery(
	item *T,
	ctx context.Context,
	tx bun.IDB,
	id ID,
	args GetterArgs,
) *bun.SelectQuery {
	// Base query by ID
	query := tx.NewSelect().Model(item).Where(fmt.Sprintf("%s = ?", b.idField), id)

	// Apply additional conditions if provided
	for k, v := range args {
		query = query.Where(k, v)
	}
	return query.Limit(1)
}

func (b BaseGetter[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	var item T
	err := b.baseQuery(&item, ctx, tx, id, args).Scan(ctx)
	isNoRows := errors.Is(err, sql.ErrNoRows)

	switch {
	case err != nil && !isNoRows:
		return nil, err
	case b.strict && isNoRows:
		return nil, sql.ErrNoRows
	case isNoRows:
		return nil, nil
	default:
		return &item, nil
	}
}

func NewBaseGetter[T any, ID pkg.IDLike](strict bool, idField string) *BaseGetter[T, ID] {
	if idField == "" {
		idField = "id"
	}
	return &BaseGetter[T, ID]{strict: strict, idField: idField}
}

type GetterWithDeleted[T any, ID pkg.IDLike] struct {
	BaseGetter[T, ID]
}

func (g *GetterWithDeleted[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	var item T
	query := g.BaseGetter.baseQuery(&item, ctx, tx, id, args).Where("deleted_at IS NULL")
	if err := query.WhereAllWithDeleted().Scan(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

func NewGetterWithDeleted[T any, ID pkg.IDLike](strict bool, idField string) *GetterWithDeleted[T, ID] {
	return &GetterWithDeleted[T, ID]{BaseGetter[T, ID]{strict: strict, idField: idField}}
}

// CustomFieldGetter is a generic type used for querying custom fields and optionally loading specified relations.
// T represents the data model type, and ID represents the custom field type, constrained by the IDLike interface.
// The `field` property specifies the custom field to be queried.
// The `relations` slice defines a list of relations to include when loading the data.
type CustomFieldGetter[T any, ID pkg.IDLike] struct {
	strict    bool
	field     string
	relations []string
}

func (c *CustomFieldGetter[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	var item T
	query := NewBaseGetter[T, ID](c.strict, c.field).baseQuery(&item, ctx, tx, id, args)
	for _, relation := range c.relations {
		query = query.Relation(relation)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

func NewCustomFieldGetter[T any, ID pkg.IDLike](
	strict bool,
	field string,
	relations ...string,
) *CustomFieldGetter[T, ID] {
	return &CustomFieldGetter[T, ID]{strict: strict, field: field, relations: relations}
}

type RequireArgsGetter[T any, ID pkg.IDLike] struct {
	idField string
	strict  bool
	args    []string
}

func (r *RequireArgsGetter[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing required args: %v", r.args)
	}
	for _, arg := range r.args {
		if _, ok := args[arg]; !ok {
			return nil, fmt.Errorf("missing required arg %q", arg)
		}
	}
	return NewBaseGetter[T, ID](r.strict, r.idField).Get(ctx, tx, id, args)
}

func NewRequireArgsGetter[T any, ID pkg.IDLike](strict bool, idField string, args ...string) *RequireArgsGetter[T, ID] {
	return &RequireArgsGetter[T, ID]{strict: strict, args: args, idField: idField}
}

var ErrSavedObjectNotFound = errors.New("the saved object was not found")

type BaseSaver[T any, ID pkg.IDLike] struct {
	strict bool
}

func (b BaseSaver[T, ID]) Save(ctx context.Context, tx bun.IDB, item *T) error {
	res, err := tx.NewInsert().Model(item).Exec(ctx)
	if mberr.IsUniqueError(err) {
		return fmt.Errorf("duplicate key: %w", err)
	}
	if mberr.IsForeignKeyError(err) {
		return fmt.Errorf("foreign key violation: %w", err)
	}
	if mberr.IsNotNullError(err) {
		return fmt.Errorf("not null violation: %w", err)
	}
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if b.strict && count == 0 {
		return ErrSavedObjectNotFound
	}
	return err
}

func NewBaseSaver[T any, ID pkg.IDLike](strict bool) *BaseSaver[T, ID] {
	return &BaseSaver[T, ID]{strict: strict}
}

var ErrUpdatedObjectNotFound = errors.New("the updated object was not found")

type BaseUpdater[T any, ID pkg.IDLike] struct {
	strict bool
}

func (b BaseUpdater[T, ID]) Update(ctx context.Context, tx bun.IDB, id ID, item *T, columns ...string) error {
	q := tx.NewUpdate().Model(item).WherePK()
	if len(columns) < 1 {
		l.Log.Warn().Type("object-type", item).Msg("BaseUpdater.Update: empty columns list")
		return nil
	}
	q = q.Column(columns...)
	affected, err := q.Exec(ctx)
	if err != nil {
		return err
	}
	count, err := affected.RowsAffected()
	if err != nil {
		return err
	}
	if b.strict && count == 0 {
		return ErrUpdatedObjectNotFound
	}
	return err
}

func NewBaseUpdater[T any, ID pkg.IDLike](strict bool) *BaseUpdater[T, ID] {
	return &BaseUpdater[T, ID]{strict: strict}
}

type CustomFieldUpdater[T any, ID pkg.IDLike] struct {
	strict bool
	field  string
}

func (c *CustomFieldUpdater[T, ID]) Update(ctx context.Context, tx bun.IDB, id ID, item *T, columns ...string) error {
	q := tx.NewUpdate().Model(item).Where(fmt.Sprintf("%s = ?", c.field), id)
	if len(columns) > 0 {
		q = q.Column(columns...)
	}
	affected, err := q.Exec(ctx)
	if err != nil {
		return err
	}
	count, err := affected.RowsAffected()
	if err != nil {
		return err
	}
	if c.strict && count == 0 {
		return ErrUpdatedObjectNotFound
	}
	return err
}

func NewCustomFieldUpdater[T any, ID pkg.IDLike](strict bool, field string) *CustomFieldUpdater[T, ID] {
	return &CustomFieldUpdater[T, ID]{strict: strict, field: field}
}

var ErrDeletedObjectNotFound = errors.New("the deleted object was not found")

type BaseDeleter[T any, ID pkg.IDLike] struct {
	strict  bool
	idField string
}

func (b BaseDeleter[T, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	affected, err := tx.NewDelete().Model((*T)(nil)).Where(fmt.Sprintf("%s = ?", b.idField), id).Exec(ctx)
	if err != nil {
		return err
	}
	count, err := affected.RowsAffected()
	if err != nil {
		return err
	}
	if b.strict && count == 0 {
		return ErrDeletedObjectNotFound
	}
	return err
}

func NewBaseDeleter[T any, ID pkg.IDLike](strict bool) *BaseDeleter[T, ID] {
	return &BaseDeleter[T, ID]{strict: strict, idField: "id"}
}

func NewDeleterWithIdField[T any, ID pkg.IDLike](strict bool, idField string) *BaseDeleter[T, ID] {
	return &BaseDeleter[T, ID]{strict: strict, idField: idField}
}

type BaseBulkDeleter[T any] struct {
	strict bool
}

func (b *BaseBulkDeleter[T]) BulkDelete(ctx context.Context, tx bun.IDB, args GetterArgs) error {
	if len(args) == 0 {
		return fmt.Errorf("mass deletion without filtering arguments is not allowed")
	}
	query := tx.NewDelete().Model((*T)(nil))
	for k, v := range args {
		query = query.Where(k, v)
	}
	affected, err := query.Exec(ctx)
	if err != nil {
		return err
	}
	count, err := affected.RowsAffected()
	if err != nil {
		return err
	}
	if b.strict && count == 0 {
		return ErrDeletedObjectNotFound
	}
	return nil
}

func NewBaseBulkDeleter[T any](strict bool) *BaseBulkDeleter[T] {
	return &BaseBulkDeleter[T]{strict: strict}
}

type CustomFieldDeleter[T any, ID pkg.IDLike] struct {
	strict bool
	field  string
}

type BaseLister[T any, ID pkg.IDLike] struct {
	applyFilters []func(q *bun.SelectQuery) *bun.SelectQuery
}

func (b *BaseLister[T, ID]) List(ctx context.Context, tx bun.IDB, f filters.ListFilter) (
	filters.ListResponse[T],
	error,
) {
	var items []T
	q := tx.NewSelect().Model(&items)
	q = f.Apply(q)
	for _, af := range b.applyFilters {
		q = af(q)
	}
	total, err := q.ScanAndCount(ctx)
	if err != nil {
		return filters.ListResponse[T]{}, err
	}
	return filters.ListResponse[T]{Items: items, Total: total, Limit: f.GetLimit(), Page: f.GetPage()}, nil
}

func (b *BaseLister[T, ID]) WithFilter(f func(q *bun.SelectQuery) *bun.SelectQuery) *BaseLister[T, ID] {
	b.applyFilters = append(b.applyFilters, f)
	return b
}

func NewBaseLister[T any, ID pkg.IDLike]() *BaseLister[T, ID] {
	return &BaseLister[T, ID]{}
}

// BaseRepository provides the default implementation of Repository using bun ORM.
type BaseRepository[T any, ID pkg.IDLike] struct{}

// Save inserts an entity into the database.
func (r *BaseRepository[T, ID]) Save(ctx context.Context, tx bun.IDB, item *T) error {
	return NewBaseSaver[T, ID](false).Save(ctx, tx, item)
}

// Get fetches an entity by ID.
func (r *BaseRepository[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	return NewBaseGetter[T, ID](false, "id").Get(ctx, tx, id, args)
}

// List fetches entities using a filter.
func (r *BaseRepository[T, ID]) List(ctx context.Context, tx bun.IDB, f filters.ListFilter) (
	filters.ListResponse[T],
	error,
) {
	return NewBaseLister[T, ID]().List(ctx, tx, f)
}

// Update modifies an entity.
func (r *BaseRepository[T, ID]) Update(ctx context.Context, tx bun.IDB, id ID, item *T, columns ...string) error {
	return NewBaseUpdater[T, ID](false).Update(ctx, tx, id, item, columns...)
}

// Delete removes an entity by ID.
func (r *BaseRepository[T, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	return NewBaseDeleter[T, ID](false).Delete(ctx, tx, id)
}

func NewBaseRepository[T any, ID pkg.IDLike]() *BaseRepository[T, ID] {
	return &BaseRepository[T, ID]{}
}

type BaseFinder[T any] struct {
	allowed map[string]struct{} // whitelist of allowed fields
}

// NewBaseFinder creates a new BaseFinder with allowed fields.
func NewBaseFinder[T any](allowed []string) *BaseFinder[T] {
	m := make(map[string]struct{}, len(allowed))
	for _, f := range allowed {
		m[f] = struct{}{}
	}
	return &BaseFinder[T]{allowed: m}
}

// GetByField fetches an entity by a whitelisted field.
func (f *BaseFinder[T]) GetByField(ctx context.Context, tx bun.IDB, field string, value any) (*T, error) {
	// safety: only whitelisted fields
	if _, ok := f.allowed[field]; !ok {
		return nil, fmt.Errorf("field %q not allowed", field)
	}

	var item T
	err := tx.NewSelect().
		Model(&item).
		Where(fmt.Sprintf("%s = ?", field), value).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

type repository[T any, ID pkg.IDLike] struct {
	getter  Getter[T, ID]
	saver   Saver[T]
	deleter Deleter[ID]
	lister  Lister[T]
	updater Updater[T, ID]
}

func (r *repository[T, ID]) Save(ctx context.Context, tx bun.IDB, item *T) error {
	return r.saver.Save(ctx, tx, item)
}

func (r *repository[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	return r.getter.Get(ctx, tx, id, args)
}

func (r *repository[T, ID]) List(ctx context.Context, tx bun.IDB, filter filters.ListFilter) (
	filters.ListResponse[T],
	error,
) {
	return r.lister.List(ctx, tx, filter)
}

func (r *repository[T, ID]) Update(ctx context.Context, tx bun.IDB, id ID, item *T, columns ...string) error {
	return r.updater.Update(ctx, tx, id, item, columns...)
}

func (r *repository[T, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	return r.deleter.Delete(ctx, tx, id)
}

type Builder[T any, ID pkg.IDLike] struct {
	getter  Getter[T, ID]
	saver   Saver[T]
	deleter Deleter[ID]
	lister  Lister[T]
	updater Updater[T, ID]
}

func (b *Builder[T, ID]) WithGetter(g Getter[T, ID]) *Builder[T, ID] {
	b.getter = g
	return b
}

func (b *Builder[T, ID]) WithSaver(s Saver[T]) *Builder[T, ID] {
	b.saver = s
	return b
}

func (b *Builder[T, ID]) WithDeleter(d Deleter[ID]) *Builder[T, ID] {
	b.deleter = d
	return b
}

func (b *Builder[T, ID]) WithLister(l Lister[T]) *Builder[T, ID] {
	b.lister = l
	return b
}

func (b *Builder[T, ID]) WithUpdater(u Updater[T, ID]) *Builder[T, ID] {
	b.updater = u
	return b
}

func (b *Builder[T, ID]) Build() Repository[T, ID] {
	if b.getter == nil {
		b.getter = NewBaseGetter[T, ID](false, "id")
	}
	if b.saver == nil {
		b.saver = NewBaseSaver[T, ID](false)
	}
	if b.deleter == nil {
		b.deleter = NewBaseDeleter[T, ID](false)
	}
	if b.lister == nil {
		b.lister = NewBaseLister[T, ID]()
	}
	if b.updater == nil {
		b.updater = NewBaseUpdater[T, ID](false)
	}
	return &repository[T, ID]{getter: b.getter, saver: b.saver, deleter: b.deleter, lister: b.lister, updater: b.updater}
}

func NewRepositoryBuilder[T any, ID pkg.IDLike]() *Builder[T, ID] {
	return &Builder[T, ID]{}
}

type OnConflictSaver[T any, ID pkg.IDLike] struct {
	onConflictExpression string
}

func (o *OnConflictSaver[T, ID]) Save(ctx context.Context, tx bun.IDB, item *T) error {
	_, err := tx.NewInsert().Model(item).On(o.onConflictExpression).Exec(ctx)
	return err
}

func NewOnConflictSaver[T any, ID pkg.IDLike](onConflictExpression string) *OnConflictSaver[T, ID] {
	return &OnConflictSaver[T, ID]{onConflictExpression: onConflictExpression}
}

type GetterWithRelations[T any, ID pkg.IDLike] struct {
	table     string
	idField   string
	relations []string
}

func (g *GetterWithRelations[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	var item T
	query := NewBaseGetter[T, ID](true, fmt.Sprintf(`"%s"."%s"`, g.table, g.idField)).baseQuery(&item, ctx, tx, id, args)
	for _, relation := range g.relations {
		query = query.Relation(relation)
	}
	err := query.Limit(1).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func NewGetterWithRelations[T any, ID pkg.IDLike](
	table, idField string,
	relations ...string,
) *GetterWithRelations[T, ID] {
	return &GetterWithRelations[T, ID]{table: table, idField: idField, relations: relations}
}

type ListerWithRelations[T any, ID pkg.IDLike] struct {
	relations []string
}

func (b ListerWithRelations[T, ID]) List(
	ctx context.Context,
	tx bun.IDB,
	f filters.ListFilter,
) (filters.ListResponse[T], error) {
	var items []T
	q := tx.NewSelect().Model(&items)
	for _, relation := range b.relations {
		q = q.Relation(relation)
	}
	total, err := f.Apply(q).ScanAndCount(ctx)
	if err != nil {
		return filters.ListResponse[T]{}, err
	}
	return filters.ListResponse[T]{Items: items, Total: total, Limit: f.GetLimit(), Page: f.GetPage()}, nil
}

func NewListerWithRelations[T any, ID pkg.IDLike](relations ...string) ListerWithRelations[T, ID] {
	return ListerWithRelations[T, ID]{relations: relations}
}

var (
	_ Getter[any, string] = (*GetterWithRelations[any, string])(nil)
)
