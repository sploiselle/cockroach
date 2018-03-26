// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import _ from "lodash";
import React from "react";

// import * as catalog from "src/views/reports/containers/charts/catalog";
import { cockroach } from "src/js/protos";

import "./chartCatalog.styl";
// import { ChartSection } from "../catalog";

function RenderListCell(props: {
  links: cockroach.ts.catalog.IndividualChart[];
}) {
  return (
    <div>
      <ul>
        {_.map(props.links, link => (
          <li key={link.collectionTitle}>
            <a
              className="catalog-link"
              href={"/#/debug/chart-catalog/" + link.collectionTitle}
            >
              {link.title}
            </a>
          </li>
        ))}
      </ul>
    </div>
  );
}

function RenderSectionTable(props: {
  section: cockroach.ts.catalog.ChartSection;
  key: number;
}) {
  if(props.section.subsections.length == 0 && props.section.charts.length == 0){
    return ""
  }
  return (
    <div>
      <table
        className={"inner-table catalog-table-level-" + props.section.level}
      >
        <tr>
          <th className="catalog-table__header">
            <a
              className="catalog-link"
              key={props.key}
              href={"/#/debug/chart-catalog/" + props.section.collectionTitle}
            >
              {props.section.title}
            </a>
          </th>
          <td className="catalog-table__cell">
            <RenderListCell
              links={
                props.section.charts as cockroach.ts.catalog.IndividualChart[]
              }
            />
            {_.map(props.section.subsections, (sec, key) => (
              <RenderSectionTable
                section={sec as cockroach.ts.catalog.ChartSection}
                key={key}
              />
            ))}
          </td>
        </tr>
      </table>
    </div>
  );
}

function RenderSection(props: {
  section: cockroach.ts.catalog.ChartSection;
  key: number;
}) {
  function createMarkup() {
    return { __html: props.section.description };
  }

  return (
    <div className="catalog-section">
      <table className="catalog-table">
        <tr>
          <td className="catalog-table__section-header">
            <div className="sticky">
              <h2 className="section-title">{props.section.title}</h2>
            </div>
            <p
              className="section-description"
              dangerouslySetInnerHTML={createMarkup()}
            />
          </td>
          <td>
            <div className="catalog-table__chart-links">
              {_.map(props.section.subsections, (sec, key) => (
                <RenderSectionTable
                  section={sec as cockroach.ts.catalog.ChartSection}
                  key={key}
                />
              ))}
            </div>
          </td>
        </tr>
      </table>
    </div>
  );
}

type ChartCatalogState = {
  error: Error;
  isLoaded: boolean;
  isPopulated: boolean;
  catalog: cockroach.ts.catalog.ChartSection[];
};

export default class ChartCatalog extends React.Component<
  {},
  ChartCatalogState
> {
  constructor(props) {
    super(props);
    this.state = {
      error: null,
      isLoaded: false,
      catalog: []
    };
  }

  componentDidMount() {
    fetch("/_admin/v1/chartcatalog")
      .then(res => res.json())
      .then(
        result => {
          this.setState({
            isLoaded: true,
            catalog: result.catalog
          });
        },
        error => {
          this.setState({
            isLoaded: true,
            error
          });
        }
      );
    window.scrollTo(0, 0);
  }

  render() {
    const { error, isLoaded, catalog } = this.state;
    if (error) {
      return <div>Error: {error.message}</div>;
    } else if (!isLoaded) {
      return <div>Loading...</div>;
    } else {
      return (
        <section className="section">
          <h1>Chart Catalog</h1>
          <p className="catalog-description">
            Understand your cluster's behavior with this curated set of charts.
            <br />
            <br />
            <strong className="catalog-link">Bold</strong> links return all •{" "}
            <span className="catalog-link">Charts</span> under their{" "}
            <span className="overhang">overhang.</span>
          </p>
          <div className="catalog">
            {_.map(catalog, (sec, key) => (
              <RenderSection section={sec} key={key} />
            ))}
          </div>
        </section>
      );
    }
  }
}
