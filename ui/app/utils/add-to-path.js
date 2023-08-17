/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// Adds a string to the end of a URL path while being mindful of query params
export default function addToPath(url, extension = '', additionalParams) {
  const [path, params] = url.split('?');
  let newUrl = `${path}${extension}`;

  if (params) {
    newUrl += `?${params}`;
  }

  if (additionalParams) {
    if (params) {
      newUrl += `&${additionalParams}`;
    } else {
      newUrl += `?${additionalParams}`;
    }
  }

  return newUrl;
}
