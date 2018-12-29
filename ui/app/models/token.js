import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';

const { computed } = Ember;

export default Model.extend({
  secret: attr('string'),
  name: attr('string'),
  global: attr('boolean'),
  createTime: attr('date'),
  type: attr('string'),
  policies: hasMany('policy'),
  policyNames: attr(),

  accessor: computed.alias('id'),
});
