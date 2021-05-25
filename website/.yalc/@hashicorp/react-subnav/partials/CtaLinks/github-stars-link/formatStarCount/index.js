/*
 * Given:
 * starCount (int)
 * Return:
 * Formatted string, to match GitHub's typical display of star counts,
 * that is, expressed as thousands of stars
 * Or returns false for falsy starCount values
 */
function formatStarCount(starCount) {
  if (!starCount || starCount <= 0) return false
  if (starCount < 1000) return `${starCount}`
  const thousands = Math.floor(starCount / 100.0) / 10.0
  if (starCount < 100000) return `${thousands.toFixed(1)}k`
  return `${Math.floor(thousands)}k`
}

export default formatStarCount
