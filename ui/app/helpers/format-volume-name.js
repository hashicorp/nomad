import Helper from '@ember/component/helper';

/**
 * Volume Name Formatter
 *
 * Usage: {{format-volume-name source=string isPerAlloc=boolean volumeExtension=string}}
 *
 * Outputs a title/link for volumes that are per_alloc-aware.
 * (when a volume is per_alloc, its route location requires an additional extension)
 */
function formatVolumeName(_, { source, isPerAlloc, volumeExtension }, b, c) {
  return `${source}${isPerAlloc ? volumeExtension : ''}`;
}

export default Helper.helper(formatVolumeName);
