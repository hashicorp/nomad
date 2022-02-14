import classic from 'ember-classic-decorator';
import { default as ApplicationAdapter, namespace } from './application';

@classic
export default class PolicyAdapter extends ApplicationAdapter {
  namespace = namespace + '/acl';
}
