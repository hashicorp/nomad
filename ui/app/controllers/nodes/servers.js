import Ember from 'ember';

const { Controller, computed } = Ember;

export default Controller.extend({
  nodes: computed.alias('model.nodes'),
  agents: computed.alias('model.agents'),
});
