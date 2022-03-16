import { bool, equal } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';
import Fragment from 'ember-data-model-fragments/fragment';

export default class EvaluationStub extends Fragment {
  @shortUUIDProperty('evalId') shortId;
  @attr('string') evalId;
  @attr('string') type;
  @attr('string') triggeredBy;
  @attr('string') status;
  @attr('string') statusDescription;

  @belongsTo('job') job;
  @belongsTo('node') node;

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
