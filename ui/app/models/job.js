import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export default Model.extend({
  region: attr('string'),
  name: attr('string'),
  type: attr('string'),
  priority: attr('number'),
  allAtOnce: attr('boolean'),

  status: attr('string'),
  statusDescription: attr('string'),
  createIndex: attr('number'),
  modifyIndex: attr('number'),

  periodic: attr('boolean'),
  parameterized: attr('boolean'),

  // data centers (hasMany)
  // constraints (model fragment? hasMany)
  // task groups (model fragment? embedded record? hasMany)
  // tasks (model fragment? hasMany)
  // stagger (from update) number
  // max parallel (from update) number
});
