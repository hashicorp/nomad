import { default as ApplicationAdapter, namespace } from './application';
import classic from 'ember-classic-decorator';

@classic
export default class PolicyAdapter extends ApplicationAdapter {
  namespace = namespace + '/acl';
}
