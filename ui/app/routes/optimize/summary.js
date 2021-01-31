import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class OptimizeSummaryRoute extends Route {
  breadcrumbs(model) {
    if (!model) return [];

    return [
      {
        label: model.slug.replace('/', ' / '),
        args: ['optimize.summary', model.slug],
      },
    ];
  }

  async model({ jobNamespace, slug }) {
    const model = this.modelFor('optimize').find(
      summary => summary.slug === slug && summary.jobNamespace === jobNamespace
    );

    if (!model) {
      const error = new Error(`Unable to find summary for ${slug} in namespace ${jobNamespace}`);
      error.code = 404;
      notifyError(this)(error);
    } else {
      return model;
    }
  }
}
