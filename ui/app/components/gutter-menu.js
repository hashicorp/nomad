import Ember from 'ember';

const { Component, inject } = Ember;

export default Component.extend({
  system: inject.service(),

  onNamespaceChange() {},
});
