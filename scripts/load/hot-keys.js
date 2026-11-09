import http from 'k6/http';

export const options = {
    vus: 50,
    duration: '30s',
};

export default function () {
    const name = Math.random() < 0.9
        ? 'goku'
        : `fighter-${Math.floor(Math.random() * 1000)}`;

    http.get(`http://localhost:8080/fighters/${name}`);
}