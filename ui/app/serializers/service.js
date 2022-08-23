import ApplicationSerializer from './application';

export default class ServiceSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.AllocationID = hash.AllocID; // TODO: keyForRelationship maybe?
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);
    return super.normalize(typeHash, hash);
  }
}
