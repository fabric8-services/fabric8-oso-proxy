FROM centos:7
MAINTAINER "Aslak Knutsen <aslak@redhat.com>"
ENV LANG=en_US.utf8
ENV GOROOT=/tmp/go1.10
ENV PATH=$PATH:/tmp/go/bin:$GOROOT/bin

# Some packages might seem weird but they are required by the RVM installer.
RUN yum --enablerepo=centosplus install -y \
      findutils \
      git \
      make \
      mercurial \
      procps-ng \
      tar \
      wget \
      which \
      gcc \
    && yum clean all \
    && rm -rf /var/cache/yum

# Get custom go v
RUN cd /tmp \
    && wget https://storage.googleapis.com/golang/go1.10.4.linux-amd64.tar.gz  \
    && echo "fa04efdb17a275a0c6e137f969a1c4eb878939e91e1da16060ce42f02c2ec5ec go1.10.4.linux-amd64.tar.gz" > checksum \
    && sha256sum -c checksum \
    && tar xvzf go*.tar.gz \
    && mv go $GOROOT

ENTRYPOINT ["/bin/bash"]
