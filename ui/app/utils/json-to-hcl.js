/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Convert a JSON object to an HCL string.
 * The Nomad API returns VariableFlags as an object where all values are strings.
 * Some strings contain JSON-encoded arrays/objects (e.g., '["dc1"]'), while others are plain strings (e.g., 'my-app'). This function converts them to HCL format:
 * - JSON arrays/objects: preserved as-is (e.g., datacenters=["dc1"])
 * - Everything else: quoted (e.g., app_name="my-app")
 * @param {Object} obj - The JSON object to convert.
 * @returns {string} The HCL string representation of the JSON object.
 */
export default function jsonToHcl(obj) {
  if (!obj || typeof obj !== 'object') {
    return '';
  }

  const hclLines = Object.entries(obj)
    .filter(([, value]) => value != null)
    .map(([key, value]) => {
      const hclValue =
        typeof value === 'string' ? convertStringValue(value) : value; // Defensive: handle non-string values

      return `${key}=${hclValue}`;
    });

  return hclLines.length > 0 ? hclLines.join('\n') + '\n' : '';
}

/**
 * Convert a string value to HCL format.
 * - If it's a valid JSON object/array, keep as-is
 * - Otherwise, quote it
 */
function convertStringValue(value) {
  try {
    const parsed = JSON.parse(value);
    // Only preserve objects and arrays
    if (typeof parsed === 'object' && parsed !== null) {
      return value;
    }
    // For JSON primitives (numbers, booleans, null), quote them
    return JSON.stringify(value);
  } catch {
    // Not valid JSON, treat as plain string and quote it
    return JSON.stringify(value);
  }
}
