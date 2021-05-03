export const ALERT_BANNER_ACTIVE = true

// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  linkText: 'Read the blog',
  url: 'https://www.hashicorp.com/blog/announcing-hashicorp-nomad-1-1-beta',
  tag: 'New Release',
  text:
    'Announcing HashiCorp Nomad 1.1 beta with 10+ new features for greater scheduling flexibility and simplified operator experience.',
  // Set the `expirationDate prop with a datetime string (e.g. `2020-01-31T12:00:00-07:00`)
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: '2021-05-20T09:00:00-07:00',
}
