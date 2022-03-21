import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';

export default class EvaluationStub extends Fragment {
  //   @attr('string') id;
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
