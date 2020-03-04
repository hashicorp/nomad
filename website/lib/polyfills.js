// If you run into issues with features missing in IE11, you likely need to
// make additions to this file for those features.
// See https://github.com/zloirock/core-js
import 'core-js/fn/array'
import 'core-js/fn/object/assign'
import 'core-js/fn/string/ends-with'
import 'core-js/fn/string/includes'
import 'core-js/fn/string/repeat'
import 'core-js/fn/string/starts-with'
import 'core-js/fn/symbol'

/* NodeList.forEach */
if (window.NodeList && !NodeList.prototype.forEach) {
  NodeList.prototype.forEach = Array.prototype.forEach
}

/* Element.matches */
if (!Element.prototype.matches) {
  Element.prototype.matches =
    Element.prototype.msMatchesSelector ||
    Element.prototype.webkitMatchesSelector
}

/* Element.closest */
if (!Element.prototype.closest) {
  Element.prototype.closest = function(s) {
    var el = this

    do {
      if (el.matches(s)) return el
      el = el.parentElement || el.parentNode
    } while (el !== null && el.nodeType === 1)
    return null
  }
}
