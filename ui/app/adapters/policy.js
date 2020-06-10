import { default as ApplicationAdapter, namespace } from './application';

export default class Policy extends ApplicationAdapter {
  namespace = namespace + '/acl';
}
