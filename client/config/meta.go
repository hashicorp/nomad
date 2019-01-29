package config

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/structs"
)

func uniqueKeys(maps ...map[string]string) []string {
	var keys []string
	seen := make(map[string]bool)

	for _, m := range maps {
		for k := range m {
			if _, ok := seen[k]; ok {
				continue
			}

			seen[k] = true
			keys = append(keys, k)
		}
	}

	return keys
}

// ApplyMetadataDiff takes a meta map, and a slice of diffs, and returns the
// result of applying the diffs to the map.
// The input meta map will win all conflicts. This allows operators to easily
// overrule API updated configuration when updating client agents.
func ApplyMetadataDiff(logger hclog.Logger, m map[string]string, diffs []*structs.MetadataDiff) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}

	for _, entry := range diffs {
		mVal, mContains := m[entry.Key]

		switch entry.Type {
		case structs.MetadataDiffTypeAdd:
			// If the config now contains the given key, it takes precedent over the
			// previous new value.
			if mContains {
				continue
			}

			// The original map still doesn't contain this key, so we insert it.
			out[entry.Key] = entry.To

		case structs.MetadataDiffTypeRemove:
			// Entry has updated in config, so we take the new value. Otherwise we do
			// nothing.
			if !mContains || entry.From != mVal {
				continue
			}

			delete(out, entry.Key)

		case structs.MetadataDiffTypeUpdate:
			// The value has been _updated_ in the config, so we take the new value.
			if mContains && mVal != entry.From {
				continue
			}

			// The value has been removed from config, so we drop the value
			if !mContains {
				continue
			}

			// The valuehas not changed so we apply the diff value instead.
			out[entry.Key] = entry.To

		default:
			logger.Error("Cannot apply metadata diff", "type", entry.Type, "key", entry.Key, "from", entry.From, "to", entry.To)
		}
	}

	return out
}

// ComputeMetadataDiff takes two client meta maps and returns the diff between
// them. The diff is not guaranteed to be returned in a stable order.
func ComputeMetadataDiff(original, current map[string]string) []*structs.MetadataDiff {
	diff := make([]*structs.MetadataDiff, 0)

	keys := uniqueKeys(original, current)
	for _, key := range keys {
		cVal, inCurrent := current[key]
		oVal, inOriginal := original[key]

		if inCurrent && inOriginal {
			if oVal == cVal {
				continue // No diff
			}

			// Updated value
			diff = append(diff, &structs.MetadataDiff{
				Type: structs.MetadataDiffTypeUpdate,
				Key:  key,
				From: oVal,
				To:   cVal,
			})
		} else if inCurrent && !inOriginal {
			// New Key
			diff = append(diff, &structs.MetadataDiff{
				Type: structs.MetadataDiffTypeAdd,
				Key:  key,
				To:   cVal,
			})

		} else if !inCurrent && inOriginal {
			// Removed Key
			diff = append(diff, &structs.MetadataDiff{
				Type: structs.MetadataDiffTypeRemove,
				Key:  key,
				From: oVal,
			})
		}
	}

	return diff
}
