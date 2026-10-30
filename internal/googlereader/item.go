// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package googlereader

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Long-form item ID prefix and format.
const (
	ItemIDPrefix = "tag:google.com,2005:reader/item/"
	ItemIDFormat = "tag:google.com,2005:reader/item/%016x"
)

func convertEntryIDToLongFormItemID(entryID int64) string {
	return fmt.Sprintf(ItemIDFormat, entryID)
}

// parseItemID accepts every item ID format seen in clients:
//
//	"tag:google.com,2005:reader/item/00000000148b9369" (long form, padded hex)
//	"tag:google.com,2005:reader/item/2f2"              (long form, unpadded hex; NetNewsWire)
//	"000000000000048c"                                 (padded hex without prefix; Reeder)
//	"12345"                                            (decimal; Liferea)
func parseItemID(itemIDValue string) (int64, error) {
	if hexID, ok := strings.CutPrefix(itemIDValue, ItemIDPrefix); ok {
		itemID, err := strconv.ParseInt(hexID, 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse hexadecimal item ID %s: %w", itemIDValue, err)
		}
		if itemID == 0 {
			return 0, fmt.Errorf("failed to parse hexadecimal item ID %s: item ID is zero", itemIDValue)
		}
		return itemID, nil
	}

	if len(itemIDValue) == 16 {
		if itemID, err := strconv.ParseInt(itemIDValue, 16, 64); err == nil {
			return itemID, nil
		}
	}

	itemID, err := strconv.ParseInt(itemIDValue, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse decimal item ID %s: %w", itemIDValue, err)
	}
	return itemID, nil
}

func parseItemIDsFromRequest(r *http.Request) ([]int64, error) {
	items := r.Form[paramItemIDs]
	if len(items) == 0 {
		return nil, errors.New("googlereader: no items requested")
	}

	itemIDs := make([]int64, len(items))
	for i, item := range items {
		itemID, err := parseItemID(item)
		if err != nil {
			return nil, fmt.Errorf("googlereader: failed to parse item ID %s: %w", item, err)
		}
		itemIDs[i] = itemID
	}
	return itemIDs, nil
}
