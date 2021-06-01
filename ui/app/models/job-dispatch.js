import Model from '@ember-data/model';
import { attr } from '@ember-data/model';

export default class JobDispatch extends Model {
  @attr() index;
  @attr() jobCreateIndex;
  @attr() evalCreateIndex;
  @attr() evalID;
  @attr() dispatchedJobID;
}
