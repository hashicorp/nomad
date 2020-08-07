import WatchableNamespaceIDs from './watchable-namespace-ids';

export default class JobScaleAdapter extends WatchableNamespaceIDs {
  urlForFindRecord(id, type, hash) {
    return super.urlForFindRecord(id, 'job', hash, 'scale');
  }
}
