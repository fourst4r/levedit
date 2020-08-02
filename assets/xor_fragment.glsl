#version 330 core

in vec2  vTexCoords;

out vec4 fragColor;

uniform vec4 uTexBounds;
uniform sampler2D uTexture;

void main() {
	vec2 t = (vTexCoords - uTexBounds.xy) / uTexBounds.zw;

	vec4 pos = texture(uTexture, t);

	vec4 color = vec4(vec3(1.0) - pos.rgb, pos.a);
	fragColor = color;
}