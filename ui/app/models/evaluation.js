import { bool, equal } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr, belongsTo } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';

export default class Evaluation extends Model {
  @shortUUIDProperty('id') shortId;
  @shortUUIDProperty('nodeId') shortNodeId;
  @attr('number') priority;
  @attr('string') type;
  @attr('string') triggeredBy;
  @attr('string') status;
  @attr('string') statusDescription;
  @fragmentArray('placement-failure', { defaultValue: () => [] })
  failedTGAllocs;

  @bool('failedTGAllocs.length') hasPlacementFailures;
  @equal('status', 'blocked') isBlocked;

  @belongsTo('job') job;
  @belongsTo('node') node;

  @attr('number') modifyIndex;
  @attr('date') modifyTime;

  @attr('number') createIndex;
  @attr('date') createTime;

  @attr('date') waitUntil;
  @attr('string') namespace;
  @attr('string') plainJobId;

  get hasJob() {
    return !!this.plainJobId;
  }

  get hasNode() {
    return !!this.belongsTo('node').id();
  }

  get nodeId() {
    return this.belongsTo('node').id();
  }
}
