import ApplicationSerializer from './application';

export default class ServiceSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.AllocationID = hash.AllocID; // TODO: keyForRelationship maybe?
    return super.normalize(typeHash, hash);
  }
}
