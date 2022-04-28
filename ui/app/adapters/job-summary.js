import WatchableNamespaceIDs from './watchable-namespace-ids';

export default class JobSummaryAdapter extends WatchableNamespaceIDs {
  urlForFindRecord(id, type, hash) {
    return super.urlForFindRecord(id, 'job', hash, 'summary');
  }
}
