import Model, { attr } from '@ember-data/model';

export default class EvaluationStub extends Model {
  @attr('number') priority;
  @attr('string') type;
  @attr('string') triggeredBy;
  @attr('string') namespace;
  @attr('string') jobId;
  @attr('string') nodeId;
  @attr('string') deploymentId; // why doesnt evaluation have this?
  @attr('string') status;
  @attr('string') statusDescription;
  @attr('date') waitUntil;
  // next, prev, blocked
  @attr('number') modifyIndex;
  @attr('date') modifyTime;
  @attr('number') createIndex;
  @attr('date') createTime;
}
