// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package googlereader

import (
	"fmt"
	"net/http"
	"strings"

	"sss/internal/http/request"
)

// Request parameter names.
const (
	paramItemIDs         = "i"        // item IDs
	paramStreamID        = "s"        // stream to include
	paramStreamExcludes  = "xt"       // streams to exclude
	paramStreamFilters   = "it"       // streams to include
	paramStreamMaxItems  = "n"        // max items per page
	paramStreamOrder     = "r"        // sort order ("o" = oldest first)
	paramStreamStartTime = "ot"       // only items newer than this epoch
	paramStreamStopTime  = "nt"       // only items older than this epoch
	paramTagsRemove      = "r"        // tags (streams) to remove
	paramTagsAdd         = "a"        // tags (streams) to add
	paramSubscribeAction = "ac"       // subscription/edit action
	paramTitle           = "t"        // subscription title
	paramQuickAdd        = "quickadd" // URL to quick-subscribe to
	paramDestination     = "dest"     // new tag name
	paramContinuation    = "c"        // pagination offset
	paramTimestamp       = "ts"       // unix timestamp
)

// Stream ID prefixes and suffixes.
const (
	streamPrefix                 = "user/-/state/com.google/"
	userStreamPrefix             = "user/%d/state/com.google/"
	labelPrefix                  = "user/-/label/"
	userLabelPrefix              = "user/%d/label/"
	feedPrefix                   = "feed/"
	readStreamSuffix             = "read"
	starredStreamSuffix          = "starred"
	readingListStreamSuffix      = "reading-list"
	keptUnreadStreamSuffix       = "kept-unread"
	broadcastStreamSuffix        = "broadcast"
	broadcastFriendsStreamSuffix = "broadcast-friends"
	likeStreamSuffix             = "like"
)

// StreamType is the kind of a Google Reader stream.
type StreamType int

// Stream types.
const (
	NoStream StreamType = iota
	ReadStream
	StarredStream
	ReadingListStream
	KeptUnreadStream
	BroadcastStream
	BroadcastFriendsStream
	LabelStream
	FeedStream
	LikeStream
)

// String returns the stream type name.
func (st StreamType) String() string {
	switch st {
	case NoStream:
		return "NoStream"
	case ReadStream:
		return "ReadStream"
	case StarredStream:
		return "StarredStream"
	case ReadingListStream:
		return "ReadingListStream"
	case KeptUnreadStream:
		return "KeptUnreadStream"
	case BroadcastStream:
		return "BroadcastStream"
	case BroadcastFriendsStream:
		return "BroadcastFriendsStream"
	case LabelStream:
		return "LabelStream"
	case FeedStream:
		return "FeedStream"
	case LikeStream:
		return "LikeStream"
	default:
		return fmt.Sprintf("StreamType(%d)", int(st))
	}
}

// Stream is a parsed stream ID: its type and, for feeds and labels, the feed ID or label name.
type Stream struct {
	Type StreamType
	ID   string
}

func getStream(streamID string, userID int64) (Stream, error) {
	switch {
	case strings.HasPrefix(streamID, feedPrefix):
		return Stream{FeedStream, strings.TrimPrefix(streamID, feedPrefix)}, nil
	case strings.HasPrefix(streamID, fmt.Sprintf(userStreamPrefix, userID)), strings.HasPrefix(streamID, streamPrefix):
		id := strings.TrimPrefix(streamID, fmt.Sprintf(userStreamPrefix, userID))
		id = strings.TrimPrefix(id, streamPrefix)
		switch id {
		case readStreamSuffix:
			return Stream{ReadStream, ""}, nil
		case starredStreamSuffix:
			return Stream{StarredStream, ""}, nil
		case readingListStreamSuffix:
			return Stream{ReadingListStream, ""}, nil
		case keptUnreadStreamSuffix:
			return Stream{KeptUnreadStream, ""}, nil
		case broadcastStreamSuffix:
			return Stream{BroadcastStream, ""}, nil
		case broadcastFriendsStreamSuffix:
			return Stream{BroadcastFriendsStream, ""}, nil
		case likeStreamSuffix:
			return Stream{LikeStream, ""}, nil
		default:
			return Stream{}, fmt.Errorf("googlereader: unknown stream with id: %s", id)
		}
	case strings.HasPrefix(streamID, fmt.Sprintf(userLabelPrefix, userID)), strings.HasPrefix(streamID, labelPrefix):
		id := strings.TrimPrefix(streamID, fmt.Sprintf(userLabelPrefix, userID))
		id = strings.TrimPrefix(id, labelPrefix)
		return Stream{LabelStream, id}, nil
	case streamID == "":
		return Stream{NoStream, ""}, nil
	default:
		return Stream{}, fmt.Errorf("googlereader: unknown stream type: %s", streamID)
	}
}

func getStreams(streamIDs []string, userID int64) ([]Stream, error) {
	streams := make([]Stream, 0, len(streamIDs))
	for _, streamID := range streamIDs {
		s, err := getStream(streamID, userID)
		if err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}
	return streams, nil
}

type requestModifiers struct {
	ExcludeTargets []Stream
	FilterTargets  []Stream
	Streams        []Stream
	Count          int
	Offset         int
	SortDirection  string
	StartTime      int64
	StopTime       int64
	UserID         int64
}

func parseStreamFilterFromRequest(r *http.Request) (requestModifiers, error) {
	userID := request.UserID(r)
	result := requestModifiers{SortDirection: "desc", UserID: userID}
	if request.QueryStringParam(r, paramStreamOrder, "d") == "o" {
		result.SortDirection = "asc"
	}

	var err error
	if result.Streams, err = getStreams(request.QueryStringParamList(r, paramStreamID), userID); err != nil {
		return requestModifiers{}, err
	}
	if result.ExcludeTargets, err = getStreams(request.QueryStringParamList(r, paramStreamExcludes), userID); err != nil {
		return requestModifiers{}, err
	}
	if result.FilterTargets, err = getStreams(request.QueryStringParamList(r, paramStreamFilters), userID); err != nil {
		return requestModifiers{}, err
	}

	result.Count = request.QueryIntParam(r, paramStreamMaxItems, 0)
	result.Offset = request.QueryIntParam(r, paramContinuation, 0)
	result.StartTime = request.QueryInt64Param(r, paramStreamStartTime, 0)
	result.StopTime = request.QueryInt64Param(r, paramStreamStopTime, 0)
	return result, nil
}
