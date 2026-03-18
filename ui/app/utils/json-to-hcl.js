/**
 * Copyright IBM Corp. 2015, 2025
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
  if (!obj) return '';

  const hclLines = [];

  for (const key in obj) {
    const value = obj[key];
    let hclValue;

    if (typeof value === 'string') {
      //
      if (
        (value.startsWith('[') && value.endsWith(']')) ||
        (value.startsWith('{') && value.endsWith('}'))
      ) {
        hclValue = value; // Keep it as a JSON string
      } else {
        // Escape double quotes and backslashes
        hclValue = JSON.stringify(value);
      }
    } else {
      hclValue = JSON.stringify(value);
    }

    hclLines.push(`${key}=${hclValue}`);
  }

  return hclLines.join('\n');
}
