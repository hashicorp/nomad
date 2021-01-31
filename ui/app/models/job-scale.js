import Model from 'ember-data/model';
import { belongsTo } from 'ember-data/relationships';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import classic from 'ember-classic-decorator';

@classic
export default class JobSummary extends Model {
  @belongsTo('job') job;

  @fragmentArray('task-group-scale') taskGroupScales;
}
