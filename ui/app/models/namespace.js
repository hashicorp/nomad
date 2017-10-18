import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';

const { computed } = Ember;

export default Model.extend({
  name: computed.readOnly('id'),
  hash: attr('string'),
  description: attr('string'),
});
