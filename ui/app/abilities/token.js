import AbstractAbility from './abstract';
import { alias } from '@ember/object/computed';

export default class extends AbstractAbility {
  @alias('selfTokenIsManagement') canRead;
  @alias('selfTokenIsManagement') canList;
  @alias('selfTokenIsManagement') canWrite;
  @alias('selfTokenIsManagement') canUpdate;
  @alias('selfTokenIsManagement') canDestroy;
}
