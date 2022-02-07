import { helper } from '@ember/component/helper';

/**
 * CSS Class
 *
 * Usage: {{css-class updateType}}
 *
 * Outputs a css friendly class string from any human string.
 * Differs from dasherize by handling slashes.
 */
export function cssClass([updateType]) {
  /* eslint-disable-next-line ember/no-string-prototype-extensions */
  return updateType.replace(/\//g, '-').dasherize();
}

export default helper(cssClass);
