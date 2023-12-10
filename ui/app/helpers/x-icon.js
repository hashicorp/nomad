/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { inlineSvg } from 'ember-inline-svg/helpers/inline-svg';

// Generated at compile-time by ember-inline-svg
import SVGs from '../svgs';

/**
 * Icon Helper
 *
 * Usage: {{x-icon name}}
 *
 * Renders an inline svg element by looking it up at `/public/images/icons/${name}.svg`
 */
export function xIcon(params, options) {
  const name = params[0];
  const classes = [options.class, 'icon', `icon-is-${name}`]
    .compact()
    .join(' ');

  return inlineSvg(SVGs, name, { class: classes });
}

export default helper(xIcon);
