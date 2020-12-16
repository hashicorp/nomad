import Model from '@ember-data/model';
import { attr, belongsTo } from '@ember-data/model';

export default class JobVersion extends Model {
  @belongsTo('job') job;
  @attr('boolean') stable;
  @attr('date') submitTime;
  @attr('number') number;
  @attr() diff;
}
