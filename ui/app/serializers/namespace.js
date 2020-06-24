import ApplicationSerializer from './application';

export default class Namespace extends ApplicationSerializer {
  primaryKey = 'Name';
}
