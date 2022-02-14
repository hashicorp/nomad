import { default as ApplicationAdapter, namespace } from './application';

export default class PolicyAdapter extends ApplicationAdapter {
  namespace = namespace + '/acl';
}
