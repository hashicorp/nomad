/* eslint-disable ember/no-controller-access-in-routes */

// An error handler to provide to a promise catch to set an error
// on the application controller.
export default function notifyError(error) {
  this.controllerFor('application').set('error', error);
}
