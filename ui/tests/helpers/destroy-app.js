import Ember from 'ember';

export default function destroyApp(application) {
  Ember.run(application, 'destroy');
  if (window.server) {
    window.server.shutdown();
  }
}
