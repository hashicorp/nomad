/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

/**
 * Convert a JSON object to an HCL string.
 *
 * @param {Object} obj - The JSON object to convert.
 * @returns {string} The HCL string representation of the JSON object.
 */
export default function jsonToHcl(obj) {
  const hclLines = [];

  for (const key in obj) {
    const value = obj[key];
    const hclValue = typeof value === 'string' ? `"${value}"` : value;
    hclLines.push(`${key}=${hclValue}\n`);
  }

  return hclLines.join('\n');
}
