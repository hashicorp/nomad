/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Charts|Distribution Bar',
};

export const Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Standard Distribution Bar</h5>
      <div class="block" style="height:50px; width:200px;">
        <DistributionBar @data={{distributionBarData}} />
      </div>
      <p class='annotation'>The distribution bar chart proportionally show data in a single bar. It includes a tooltip out of the box, assumes the size of the container element, and is designed to be styled with CSS.</p>
      `,
    context: {
      distributionBarData: [
        { label: 'one', value: 10 },
        { label: 'two', value: 20 },
        { label: 'three', value: 30 },
      ],
    },
  };
};

export const WithClasses = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Distribution Bar With Classes</h5>
      <div class="block" style="height:50px; width:200px;">
        <DistributionBar @data={{distributionBarDataWithClasses}} />
      </div>
      <p class='annotation'>If a datum provides a <code>className</code> property, it will be assigned to the corresponding <code>rect</code> element, allowing for custom colorization.</p>
      `,
    context: {
      distributionBarDataWithClasses: [
        { label: 'Queued', value: 10, className: 'queued' },
        { label: 'Complete', value: 20, className: 'complete' },
        { label: 'Failed', value: 30, className: 'failed' },
      ],
    },
  };
};

export const Flexibility = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Distribution Bar Flexibility</h5>
      <div class="block" style="height:10px; width:600px;">
        <DistributionBar @data={{distributionBarData}} />
      </div>
      <div class="block" style="height:200px; width:30px;">
        <DistributionBar @data={{distributionBarData}} />
      </div>
      <p class='annotation'>Distribution bar assumes the dimensions of the container.</p>
      `,
    context: {
      distributionBarData: [
        { label: 'one', value: 10 },
        { label: 'two', value: 20 },
        { label: 'three', value: 30 },
      ],
    },
  };
};

export const LiveUpdating = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Live Updating Distribution Bar</h5>
      <div class="block" style="height:50px; width:600px;">
        <DistributionBar @data={{controller.distributionBarDataRotating}} />
      </div>
      <p class='annotation'>Distribution bar animates with data changes.</p>
      <div class="boxed-section">
        <div class="boxed-section-body is-dark">
          <JsonViewer @json={{controller.distributionBarDataRotating}} />
        </div>
      </div>
      `,
    context: {
      controller: Ember.Object.extend({
        timerTicks: 0,

        startTimer: Ember.on('init', function() {
          this.set(
            'timer',
            setInterval(() => {
              this.incrementProperty('timerTicks');
            }, 500)
          );
        }),

        willDestroy() {
          clearInterval(this.timer);
        },

        distributionBarDataRotating: Ember.computed('timerTicks', () => {
          return [
            { label: 'one', value: Math.round(Math.random() * 50) },
            { label: 'two', value: Math.round(Math.random() * 50) },
            { label: 'three', value: Math.round(Math.random() * 50) },
          ];
        }),
      }).create(),
    },
  };
};

export const SingleBar = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Distribution Bar Single Bar</h5>
      <div class="block" style="height:50px; width:600px;">
        <DistributionBar @data={{distributionBarDatum}} />
      </div>
      `,
    context: {
      distributionBarDatum: [{ label: 'one', value: 10 }],
    },
  };
};
