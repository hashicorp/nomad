import classic from 'ember-classic-decorator';
import ApplicationSerializer from './application';

@classic
export default class ServiceSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.AllocationID = hash.AllocID;
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);
    return super.normalize(typeHash, hash);
  }
}
