import ApplicationSerializer from './application';

export default class TaskGroupDeploymentSummary extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.PlacedCanaryAllocations = hash.PlacedCanaries || [];
    delete hash.PlacedCanaries;
    return super.normalize(typeHash, hash);
  }
}
