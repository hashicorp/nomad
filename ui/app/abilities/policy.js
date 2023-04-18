import AbstractAbility from './abstract';
import { alias } from '@ember/object/computed';
import classic from 'ember-classic-decorator';

@classic
export default class Policy extends AbstractAbility {
  @alias('selfTokenIsManagement') canRead;
  @alias('selfTokenIsManagement') canList;
  @alias('selfTokenIsManagement') canWrite;
  @alias('selfTokenIsManagement') canUpdate;
  @alias('selfTokenIsManagement') canDestroy;
}
