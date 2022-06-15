FROM gitpod/workspace-go:2022-06-15-18-10-29

RUN echo 'deb [trusted=yes] https://repo.goreleaser.com/apt/ /' | sudo tee /etc/apt/sources.list.d/goreleaser.list && \
    sudo apt-get update  && \
    sudo apt-get -y install goreleaser
