import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default class BreadcrumbsJob extends Component {
  get job() {
    return this.args.crumb.job;
  }

  @tracked parent = null;

  generateCrumb(job) {
    return {
      label: job.get('trimmedName') || job.trimmedName,
      args: [
        'jobs.job.index',
        job.get('plainId') || job.plainId,
        qpBuilder({
          jobNamespace: job.get('namespace.name') || 'default',
        }),
      ],
    };
  }

  get crumb() {
    if (!this.job) return null;
    return this.generateCrumb(this.job);
  }

  @action
  fetchParent() {
    this.parent = this.job.parent || this.job.get('parent');
    return this.generateCrumb(this.parent);
  }
}
