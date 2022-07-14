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

export default helper(function cleanKeycommand([part] /*, named*/) {
  console.log('cleaning', part);
  Object.keys(KEY_ALIAS_MAP).forEach((key) => {
    part = part.replace(key, KEY_ALIAS_MAP[key]);
  });

  return [part];
});
