import Ember from 'ember';
import Mixin from '@ember/object/mixin';
import { assert } from '@ember/debug';
import { on } from '@ember/object/evented';

export default Mixin.create({
  visibilityHandler() {
    assert('visibilityHandler needs to be overridden in the Component', false);
  },

  setupDocumentVisibility: on('init', function() {
    if (!Ember.testing) {
      this.set('_visibilityHandler', this.visibilityHandler.bind(this));
      document.addEventListener('visibilitychange', this._visibilityHandler);
    }
  }),

  removeDocumentVisibility: on('init', function() {
    if (!Ember.testing) {
      document.removeEventListener('visibilitychange', this._visibilityHandler);
    }
  }),
});
