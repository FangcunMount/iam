window.onload = function() {
  //<editor-fold desc="Changeable Configuration Block">

  const params = new URLSearchParams(window.location.search);
  const customUrl = params.get("url");
  const defaultUrl = "/openapi/authn.v2.yaml";

  const urls = [
    { name: "AuthN", url: "/openapi/authn.v2.yaml" },
    { name: "Identity", url: "/openapi/identity.v2.yaml" },
    { name: "AuthZ", url: "/openapi/authz.v2.yaml" },
    { name: "IDP", url: "/openapi/idp.v2.yaml" },
    { name: "Suggest", url: "/openapi/suggest.v2.yaml" },
  ];

  window.ui = SwaggerUIBundle({
    url: customUrl || defaultUrl,
    urls: urls,
    dom_id: '#swagger-ui',
    deepLinking: true,
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [
      SwaggerUIBundle.plugins.DownloadUrl
    ],
    layout: "StandaloneLayout"
  });

  //</editor-fold>
};
