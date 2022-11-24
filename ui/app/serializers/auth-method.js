import ApplicationSerializer from './application';

// @classic
export default class AuthMethodSerializer extends ApplicationSerializer {
  primaryKey = 'Name';
}
