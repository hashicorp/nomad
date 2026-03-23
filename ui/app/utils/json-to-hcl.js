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
  const hclLines = [];

  for (const key in obj) {
    const value = obj[key];
    let hclValue;

    if (typeof value === 'string') {
      // Try to parse as JSON to validate it's actually valid JSON
      try {
        const parsed = JSON.parse(value);
        if (typeof parsed === 'object' && parsed !== null) {
          hclValue = value;
        } else {
          hclValue = JSON.stringify(value);
        }
      } catch {
        // Not valid JSON, treat as a regular string
        hclValue = JSON.stringify(value);
      }
    } else {
      hclValue = JSON.stringify(value);
    }

    hclLines.push(`${key}=${hclValue}\n`);
  }

  return hclLines.join('\n');
}
