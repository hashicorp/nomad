/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/* eslint-disable ember/no-controller-access-in-routes */

// An error handler to provide to a promise catch to set an error
// on the application controller.
export default function notifyError(route) {
  return (error) => {
    route.controllerFor('application').set('error', error);
  };
}
