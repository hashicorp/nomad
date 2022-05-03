import * as React from 'react'
import rivetQuery from '@hashicorp/nextjs-scripts/dato/client'
import homepageQuery from './query.graphql'
import { isInternalLink } from 'lib/utils'
import ReactHead from '@hashicorp/react-head'
import IoHomeHero from 'components/io-home-hero'
import IoHomeIntro from 'components/io-home-intro'
import IoHomeInPractice from 'components/io-home-in-practice'
import IoCardContainer from 'components/io-card-container'
import IoHomeCaseStudies from 'components/io-home-case-studies'
import IoHomeCallToAction from 'components/io-home-call-to-action'
import IoHomePreFooter from 'components/io-home-pre-footer'
import s from './style.module.css'

export default function Homepage({ data }): React.ReactElement {
  const {
    seo,
    heroHeading,
    heroDescription,
    heroCtas,
    heroCards,
    introHeading,
    introDescription,
    introFeatures,
    introVideo,
    inPracticeHeading,
    inPracticeDescription,
    inPracticeCards,
    inPracticeCtaHeading,
    inPracticeCtaDescription,
    inPracticeCtaLink,
    inPracticeCtaImage,
    useCasesHeading,
    useCasesDescription,
    useCasesCards,
    caseStudiesHeading,
    caseStudiesDescription,
    caseStudiesFeatured,
    caseStudiesLinks,
    callToActionHeading,
    callToActionDescription,
    callToActionCtas,
    preFooterHeading,
    preFooterDescription,
    preFooterCtas,
  } = data
  const _introVideo = introVideo[0]

  return (
    <>
      <ReactHead
        title={seo.title}
        description={seo.description}
        image={seo.image?.url}
        twitterCard="summary_large_image"
        pageName={seo.title}
      >
        <meta name="twitter:title" content={seo.title} />
        <meta name="twitter:description" content={seo.description} />
      </ReactHead>

      <IoHomeHero
        pattern="/img/home-hero-pattern.svg"
        brand="nomad"
        heading={heroHeading}
        description={heroDescription}
        ctas={heroCtas}
        cards={heroCards.map((card) => {
          return {
            ...card,
            cta: card.cta[0],
          }
        })}
      />

      <IoHomeIntro
        isInternalLink={isInternalLink}
        brand="nomad"
        heading={introHeading}
        description={introDescription}
        features={introFeatures}
        video={{
          youtubeId: _introVideo?.youtubeId,
          thumbnail: _introVideo?.thumbnail?.url,
          heading: _introVideo?.heading,
          description: _introVideo?.description,
          person: {
            name: _introVideo?.personName,
            description: _introVideo?.personDescription,
            avatar: _introVideo?.personAvatar?.url,
          },
        }}
      />

      <section className={s.useCases}>
        <div className={s.container}>
          <IoCardContainer
            heading={useCasesHeading}
            description={useCasesDescription}
            cardsPerRow={4}
            cards={useCasesCards.map((card) => {
              return {
                eyebrow: card.eyebrow,
                link: {
                  url: card.link,
                  type: 'inbound',
                },
                heading: card.heading,
                description: card.description,
                products: card.products,
              }
            })}
          />
        </div>
      </section>

      <IoHomeInPractice
        brand="nomad"
        pattern="/img/practice-pattern.svg"
        heading={inPracticeHeading}
        description={inPracticeDescription}
        cards={inPracticeCards.map((card) => {
          return {
            eyebrow: card.eyebrow,
            link: {
              url: card.link,
              type: 'inbound',
            },
            heading: card.heading,
            description: card.description,
            products: card.products,
          }
        })}
        cta={{
          heading: inPracticeCtaHeading,
          description: inPracticeCtaDescription,
          link: inPracticeCtaLink,
          image: inPracticeCtaImage,
        }}
      />

      <IoHomeCaseStudies
        heading={caseStudiesHeading}
        description={caseStudiesDescription}
        primary={caseStudiesFeatured}
        secondary={caseStudiesLinks}
      />

      <IoHomeCallToAction
        brand="nomad"
        heading={callToActionHeading}
        content={callToActionDescription}
        links={callToActionCtas}
      />

      <IoHomePreFooter
        brand="nomad"
        heading={preFooterHeading}
        description={preFooterDescription}
        ctas={preFooterCtas}
      />
    </>
  )
}

export async function getStaticProps() {
  const { nomadHomepage } = await rivetQuery({
    query: homepageQuery,
  })

  return {
    props: {
      data: nomadHomepage,
    },
    revalidate:
      process.env.HASHI_ENV === 'production'
        ? process.env.GLOBAL_REVALIDATE
        : 10,
  }
}
