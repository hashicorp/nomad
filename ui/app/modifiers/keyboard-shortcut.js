import { inject as service } from '@ember/service';
import Modifier from 'ember-modifier';
import { registerDestructor } from '@ember/destroyable';
import { assert } from '@ember/debug';

export default class KeyboardShortcutModifier extends Modifier {
  @service keyboard;

  /**
   * For Dynamic/iterative keyboard shortcuts, our patterns look like "Shift+0" by default
   * Do a couple things to make them more human-friendly:
   * 1. Make them 1-based, instead of 0-based
   * 2. Preface numbers 1-9 with "0" to make it so "Shift+10" doesn't trigger "Shift+1" then "0", etc.
   * ^--- stops being a good solution with 100+ row lists/tables, but a better UX than waiting for shift key-up otherwise
   *
   * @param {string[]} pattern
   */
  cleanPattern(pattern) {
    let patternNumber = pattern.length === 1 && pattern[0].match(/\d+/g);
    if (!patternNumber) {
      return pattern;
    } else {
      patternNumber = +patternNumber[0]; // regex'd string[0] to num
      patternNumber = patternNumber + 1; // first item should be Shift+1, not Shift+0
      assert(
        'Dynamic keyboard shortcuts only work up to 99 digits',
        patternNumber < 100
      );
      pattern = [`Shift+${('0' + patternNumber).slice(-2)}`]; // Shift+01, not Shift+1
    }
    return pattern;
  }

  modify(element, [eventName], { label, pattern, action }) {
    let commands = [
      {
        label,
        action,
        pattern: this.cleanPattern(pattern),
        element,
      },
    ];
    this.keyboard.addCommands(commands);
    registerDestructor(this, () => {
      this.keyboard.removeCommands(commands);
    });
  }
}
