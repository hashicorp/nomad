/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

/*
  There’s no grouping of recommendations on the server, so this
  processes a list of recommendations and groups them by task
  group.
*/

@classic
export default class RecommendationSummarySerializer extends ApplicationSerializer {
  normalizeArrayResponse(store, modelClass, payload) {
    const recommendationSerializer = store.serializerFor('recommendation');
    const RecommendationModel = store.modelFor('recommendation');

    const slugToSummaryObject = {};
    const allRecommendations = [];

    payload.forEach((recommendationHash) => {
      const slug = `${JSON.stringify([
        recommendationHash.JobID,
        recommendationHash.Namespace,
      ])}/${recommendationHash.Group}`;

      if (!slugToSummaryObject[slug]) {
        slugToSummaryObject[slug] = {
          attributes: {
            jobId: recommendationHash.JobID,
            jobNamespace: recommendationHash.Namespace,
            taskGroupName: recommendationHash.Group,
          },
          recommendations: [],
        };
      }

      slugToSummaryObject[slug].recommendations.push(recommendationHash);
      allRecommendations.push(recommendationHash);
    });

    return {
      data: Object.values(slugToSummaryObject).map((summaryObject) => {
        const latest = Math.max(
          ...summaryObject.recommendations.mapBy('SubmitTime')
        );

        return {
          type: 'recommendation-summary',
          id: summaryObject.recommendations.mapBy('ID').sort().join('-'),
          attributes: {
            ...summaryObject.attributes,
            submitTime: new Date(Math.floor(latest / 1000000)),
          },
          relationships: {
            job: {
              data: {
                type: 'job',
                id: JSON.stringify([
                  summaryObject.attributes.jobId,
                  summaryObject.attributes.jobNamespace,
                ]),
              },
            },
            recommendations: {
              data: summaryObject.recommendations.map((r) => {
                return {
                  type: 'recommendation',
                  id: r.ID,
                };
              }),
            },
          },
        };
      }),
      included: allRecommendations.map(
        (recommendationHash) =>
          recommendationSerializer.normalize(
            RecommendationModel,
            recommendationHash
          ).data
      ),
    };
  }

  normalizeUpdateRecordResponse(store, primaryModelClass, payload, id) {
    return {
      data: {
        id,
        attributes: {
          isProcessed: true,
        },
      },
    };
  }
}
