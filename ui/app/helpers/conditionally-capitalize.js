/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';

/**
 * If the condition is true, capitalize the first letter of the term.
 * Otherwise, return the term in lowercase.
 */
export function conditionallyCapitalize([term, condition]) {
  return condition
    ? `${term.charAt(0).toUpperCase()}${term.substring(1)}`
    : term.toLowerCase();
}

export default Helper.helper(conditionallyCapitalize);
