import { alias } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';

export default Model.extend({
  secret: attr('string'),
  name: attr('string'),
  global: attr('boolean'),
  createTime: attr('date'),
  type: attr('string'),
  policies: hasMany('policy'),
  policyNames: attr(),

  accessor: alias('id'),
});
