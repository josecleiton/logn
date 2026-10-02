// Fora do HTML para a CSP de /docs não precisar de 'unsafe-inline'.
window.onload = function () {
  window.ui = SwaggerUIBundle({
    url: "/docs/swagger.json",
    dom_id: "#swagger-ui",
    deepLinking: true,
  });
};
