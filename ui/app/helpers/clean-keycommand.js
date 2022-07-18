// @ts-check
import { helper } from '@ember/component/helper';

const KEY_ALIAS_MAP = {
  ArrowRight: '→',
  ArrowLeft: '←',
  ArrowUp: '↑',
  ArrowDown: '↓',
  '+': ' + ',
  // Enter: '⏎',
  // Tab: '⇥',
  // Space: '␣',
  // Shift: '⇧',
};

export default helper(function cleanKeycommand([key] /*, named*/) {
  let cleaned = key;
  Object.keys(KEY_ALIAS_MAP).forEach((k) => {
    cleaned = cleaned.replace(k, KEY_ALIAS_MAP[k]);
  });
  return cleaned;
});
