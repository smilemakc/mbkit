package models

import "github.com/smilemakc/mbkit/pkg/trackable"

type TrackableDTO[ID any] interface {
	SetTrackable(trackableType trackable.Type, id ID)
}
