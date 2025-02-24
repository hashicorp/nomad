/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import routeRedirects from './route-redirects';

export function handleRouteRedirects(transition, router) {
  const currentPath = transition.intent.url || transition.targetName;

  for (const redirect of routeRedirects) {
    let shouldRedirect = false;
    let targetPath =
      typeof redirect.to === 'function'
        ? redirect.to(currentPath)
        : redirect.to;

    switch (redirect.method) {
      case 'startsWith':
        shouldRedirect = currentPath.startsWith(redirect.from);
        break;
      case 'exact':
        shouldRedirect = currentPath === redirect.from;
        break;
      case 'pattern':
        if (redirect.pattern && redirect.pattern.test(currentPath)) {
          shouldRedirect = true;
        }
        break;
    }

    if (shouldRedirect) {
      console.warn(
        `This URL has changed. Please update your bookmark from ${currentPath} to ${targetPath}`
      );

      router.replaceWith(targetPath, {
        queryParams: transition.to.queryParams,
      });
      return true;
    }
  }
}
