import { bool, equal } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr, belongsTo } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';

export default class Evaluation extends Model {
  @shortUUIDProperty('id') shortId;
  @attr('number') priority;
  @attr('string') type;
  @attr('string') triggeredBy;
  @attr('string') status;
  @attr('string') statusDescription;
  @fragmentArray('placement-failure', { defaultValue: () => [] }) failedTGAllocs;

  @bool('failedTGAllocs.length') hasPlacementFailures;
  @equal('status', 'blocked') isBlocked;

  @belongsTo('job') job;

  @attr('number') modifyIndex;
  @attr('date') modifyTime;

  @attr('number') createIndex;
  @attr('date') createTime;

  @attr('date') waitUntil;
}
