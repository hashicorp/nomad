import { default as ApplicationAdapter, namespace } from './application';

export default ApplicationAdapter.extend({
  namespace: namespace + '/acl',
});
