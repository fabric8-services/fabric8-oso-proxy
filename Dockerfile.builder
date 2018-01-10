FROM centos:7
MAINTAINER "Aslak Knutsen <aslak@redhat.com>"
ENV LANG=en_US.utf8
ENV GOROOT=/tmp/go1.9
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
    && yum clean all \
    && rm -rf /var/cache/yum

# Get custom go v
RUN cd /tmp \
    && wget https://storage.googleapis.com/golang/go1.9.2.linux-amd64.tar.gz  \
    && tar xvzf go*.tar.gz \
    && mv go $GOROOT

ENTRYPOINT ["/bin/bash"]
