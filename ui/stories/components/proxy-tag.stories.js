/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|Proxy Tag',
};

export const ProxyTag = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Proxy Tag</h5>
      <h6 class="title is-6">Some kind of title <ProxyTag/></h6>
    `,
    context: {},
  };
};
