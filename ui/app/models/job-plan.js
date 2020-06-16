import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import { hasMany } from 'ember-data/relationships';

export default class JobPlan extends Model {
  @attr() diff;
  @fragmentArray('placement-failure', { defaultValue: () => [] }) failedTGAllocs;
  @hasMany('allocation') preemptions;
}
