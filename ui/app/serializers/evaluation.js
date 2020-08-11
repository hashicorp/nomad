import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class Evaluation extends ApplicationSerializer {
  @service system;

  mapToArray = ['FailedTGAllocs'];
  separateNanos = ['CreateTime', 'ModifyTime'];

  normalize(typeHash, hash) {
    hash.PlainJobId = hash.JobID;
    hash.Namespace =
      hash.Namespace ||
      get(hash, 'Job.Namespace') ||
      this.get('system.activeNamespace.id') ||
      'default';
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);

    return super.normalize(typeHash, hash);
  }
}
