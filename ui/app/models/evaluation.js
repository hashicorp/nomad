import { bool, equal } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';

export default Model.extend({
  shortId: shortUUIDProperty('id'),
  priority: attr('number'),
  type: attr('string'),
  triggeredBy: attr('string'),
  status: attr('string'),
  statusDescription: attr('string'),
  failedTGAllocs: fragmentArray('placement-failure', { defaultValue: () => [] }),

  hasPlacementFailures: bool('failedTGAllocs.length'),
  isBlocked: equal('status', 'blocked'),

  // TEMPORARY: https://github.com/emberjs/data/issues/5209
  originalJobId: attr('string'),

  job: belongsTo('job'),

  modifyIndex: attr('number'),

  waitUntil: attr('date'),
});
