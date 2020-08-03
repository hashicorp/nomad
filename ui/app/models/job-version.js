import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';

export default class JobVersion extends Model {
  @belongsTo('job') job;
  @attr('boolean') stable;
  @attr('date') submitTime;
  @attr('number') number;
  @attr() diff;
}
