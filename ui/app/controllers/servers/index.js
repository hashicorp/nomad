import Ember from 'ember';

const { Controller, computed, inject } = Ember;

export default Controller.extend({
  serversController: inject.controller('servers'),
  isForbidden: computed.alias('serversController.isForbidden'),
});
